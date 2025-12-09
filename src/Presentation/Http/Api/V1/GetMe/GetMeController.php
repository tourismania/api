<?php

declare(strict_types=1);
declare(ticks=1000);

namespace App\Presentation\Http\Api\V1\GetMe;

use App\Application\UseCases\GetMe\GetMeQuery;
use Nelmio\ApiDocBundle\Attribute\Security;
use OpenApi\Attributes as OA;
use Symfony\Component\DependencyInjection\Attribute\Autowire;
use Symfony\Component\HttpFoundation\JsonResponse;
use Symfony\Component\Messenger\Exception\ExceptionInterface;
use Symfony\Component\Messenger\HandleTrait;
use Symfony\Component\Messenger\MessageBusInterface;
use Symfony\Component\Routing\Attribute\Route;

class GetMeController
{
    use HandleTrait;

    private GetMeResolver $getMeResolver;

    public function __construct(
        #[Autowire(service: 'query.bus')] MessageBusInterface $messageBus, // из-за использвания $messageBus в трейтах, используем прямое назначение в конструкторе
        GetMeResolver $getMeResolver,
    ) {
        $this->messageBus = $messageBus;
        $this->getMeResolver = $getMeResolver;
    }

    /**
     * @throws ExceptionInterface
     */
    #[Route('/api/v1/me', name: 'me', methods: ['GET'])]
    #[OA\Tag(name: 'Profile')]
    #[Security(name: 'Bearer')]
    public function __invoke(): JsonResponse
    {
        $getMeDto = $this->getMeResolver->resolve();

        $result = $this->handle(new GetMeQuery(
            $getMeDto->id,
            $getMeDto->email,
            $getMeDto->phone,
            $getMeDto->firstName,
            $getMeDto->lastName,
            $getMeDto->roles
        ));

        return new JsonResponse($result);
    }
}
