<?php

declare(strict_types=1);
declare(ticks=1000);

namespace App\Presentation\Http\Api\V1\CreateUser;

use App\Application\UseCases\CreateUser\CreateUserCommand;
use Symfony\Component\HttpFoundation\JsonResponse;
use Symfony\Component\HttpKernel\Attribute\MapRequestPayload;
use Symfony\Component\Messenger\Exception\ExceptionInterface;
use Symfony\Component\Messenger\HandleTrait;
use Symfony\Component\Messenger\MessageBusInterface;
use Symfony\Component\Routing\Attribute\Route;

class CreateUserController
{
    use HandleTrait;

    public function __construct(MessageBusInterface $messageBus){
        $this->messageBus = $messageBus;
    }

    /**
     * @throws ExceptionInterface
     */
    #[Route('/api/v1/users', name: 'users_create', methods: ['POST'])]
    public function __invoke(
        #[MapRequestPayload] CreateUserDto $dto,
    ): JsonResponse
    {
        $result = $this->handle(new CreateUserCommand($dto->firstName, $dto->lastName, $dto->email));

        return new JsonResponse($result);
    }

}
