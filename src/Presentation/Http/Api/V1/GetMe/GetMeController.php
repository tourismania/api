<?php

declare(strict_types=1);
declare(ticks=1000);

namespace App\Presentation\Http\Api\V1\GetMe;

use App\Application\UseCases\GetMe\GetMeQuery;
use Symfony\Bundle\SecurityBundle\Security;
use Symfony\Component\DependencyInjection\Attribute\Autowire;
use Symfony\Component\HttpFoundation\JsonResponse;
use Symfony\Component\HttpKernel\Attribute\MapRequestPayload;
use Symfony\Component\Messenger\Exception\ExceptionInterface;
use Symfony\Component\Messenger\HandleTrait;
use Symfony\Component\Messenger\MessageBusInterface;
use Symfony\Component\Routing\Attribute\Route;

class GetMeController
{
    use HandleTrait;

    private Security $security;

    public function __construct(
        #[Autowire(service: 'query.bus')] MessageBusInterface $messageBus, // из-за использвания $messageBus в трейтах, используем прямое назначение в конструкторе
    ){
        $this->messageBus = $messageBus;
    }

    /**
     * @throws ExceptionInterface
     */
    #[Route('/api/v1/me', name: 'me', methods: ['GET'])]
    public function __invoke(
        #[MapRequestPayload(resolver: GetMeResolver::class)] GetMeDto $getMeDto
    ): JsonResponse
    {
        $result = $this->handle(new GetMeQuery(
            $getMeDto->id,
            $getMeDto->email,
            $getMeDto->phone,
            $getMeDto->firstName,
            $getMeDto->lastName,
        ));

        return new JsonResponse($result);
    }

}
