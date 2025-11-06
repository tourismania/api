<?php

declare(strict_types=1);
declare(ticks=1000);

namespace App\Presentation\Http\Api\V1\CreateUser;

use App\Application\UseCases\CreateUser\CreateUserCommandHandler;
use Symfony\Bundle\FrameworkBundle\Controller\AbstractController;
use Symfony\Component\HttpKernel\Attribute\MapRequestPayload;
use Symfony\Component\Routing\Attribute\Route;

class CreateUserController extends AbstractController
{
    #[Route('/api/v1/users', name: 'users_create', methods: ['POST'])]
    public function __invoke(
        #[MapRequestPayload] CreateUserDto $dto,
        CreateUserCommandHandler $commandHandler
    ): \Symfony\Component\HttpFoundation\JsonResponse
    {
        return $this->json($commandHandler->handle($dto));
    }

}
